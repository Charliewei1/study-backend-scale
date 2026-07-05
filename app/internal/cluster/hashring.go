// Package cluster は、将来 Redis やデータベースをシャーディングするときの
// 考え方を学ぶための小さなライブラリを提供します。
//
// 今日の時点ではサーバ本体には組み込みません。URL 短縮サービスがさらに大きく
// なったときに、「この短縮コードはどの Redis / DB シャードに置くか」を決める
// 部品として使える形にしてあります。
package cluster

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"strconv"
	"sync"
)

// HashRing は仮想ノード付きの consistent hashing ring です。
//
// 単純な shard = hash(key) % N という割り当ては、ノード数 N が変わると
// ほとんどの key の行き先が変わります。10 台から 11 台に増やすだけでも
// % 10 と % 11 の結果は別物になるため、キャッシュやシャードの中身を大きく
// 動かす必要があります。
//
// consistent hashing では、key も node も同じ円周上の位置に置きます。
// key は時計回りに見つかる最初の node に割り当てます。新しい node を足しても、
// その node の直前の区間に入った key だけが移動するので、再配置を小さくできます。
//
// replicas は 1 つの実ノードをリング上に何個の仮想ノードとして置くかです。
// 実ノードを 1 点だけ置くと、たまたま広い区間を持つ node と狭い区間の node が
// 生まれます。仮想ノードを増やすと各実ノードが円周上の多くの場所を担当するため、
// 偶然の偏りが平均化され、分布がなだらかになります。
//
// NewHashRing で作った値をコピーせず使う限り、HashRing の公開メソッドは並行利用
// できます。AddNode と RemoveNode はリングを変更するため排他ロックを取り、GetNode
// は読み取りだけなので RLock で十分です。
type HashRing struct {
	mu       sync.RWMutex
	replicas int
	nodes    map[string]struct{}
	points   []ringPoint
}

type ringPoint struct {
	hash uint64
	node string
}

// NewHashRing は replicas 個の仮想ノードを使う空の HashRing を返します。
// replicas が 0 以下の場合は、最小構成として 1 を使います。
func NewHashRing(replicas int) *HashRing {
	if replicas < 1 {
		replicas = 1
	}

	return &HashRing{
		replicas: replicas,
		nodes:    make(map[string]struct{}),
	}
}

// AddNode は実ノードをリングに追加します。
// 同じノード名を二度追加した場合は、リングを変えずにそのまま返します。
func (r *HashRing) AddNode(node string) {
	if node == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.nodes[node]; ok {
		return
	}

	r.nodes[node] = struct{}{}
	for replica := 0; replica < r.replicas; replica++ {
		r.points = append(r.points, ringPoint{
			hash: hashKey(virtualNodeKey(node, replica)),
			node: node,
		})
	}
	r.sortPoints()
}

// RemoveNode は実ノードと、そのノードに対応する仮想ノードをリングから削除します。
// 存在しないノードを指定した場合は何もしません。
func (r *HashRing) RemoveNode(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.nodes[node]; !ok {
		return
	}

	delete(r.nodes, node)
	filtered := r.points[:0]
	for _, point := range r.points {
		if point.node != node {
			filtered = append(filtered, point)
		}
	}
	r.points = filtered
}

// GetNode は key を担当する実ノードを返します。
// リングが空の場合は ok=false を返します。
func (r *HashRing) GetNode(key string) (node string, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.points) == 0 {
		return "", false
	}

	h := hashKey(key)
	idx := sort.Search(len(r.points), func(i int) bool {
		return r.points[i].hash >= h
	})
	if idx == len(r.points) {
		idx = 0
	}

	return r.points[idx].node, true
}

func (r *HashRing) sortPoints() {
	sort.Slice(r.points, func(i, j int) bool {
		if r.points[i].hash == r.points[j].hash {
			return r.points[i].node < r.points[j].node
		}
		return r.points[i].hash < r.points[j].hash
	})
}

func virtualNodeKey(node string, replica int) string {
	return node + "#" + strconv.Itoa(replica)
}

func hashKey(key string) uint64 {
	// SHA-256 は標準ライブラリだけで使える決定的なハッシュです。
	// crc32 や FNV より計算は重いですが、このパッケージは教材用なので、
	// 構造化された node 名や key でも分布が素直に見えることを優先します。
	sum := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint64(sum[:8])
}
