#!/usr/bin/env python3
"""教材ページ(.mdx)を、ターミナルで読める素のMarkdownへ落とす。

./study hint はエディタではなくターミナルで読むので、
JSXコンポーネントやStarlight記法をそのまま出すと読みにくい。
"""
import re
import sys


def unquote(value: str) -> str:
    return value.replace("\\'", "'").replace('\\"', '"')


def render_quiz(match: re.Match) -> str:
    body = match.group(1)

    def attr(name):
        m = re.search(rf'{name}="((?:[^"\\]|\\.)*)"', body)
        return unquote(m.group(1)) if m else ""

    question = attr("question")
    explanation = attr("explanation")

    choices = []
    m = re.search(r"choices=\{\[(.*?)\]\}", body, re.S)
    if m:
        choices = [unquote(c) for c in re.findall(r"'((?:[^'\\]|\\.)*)'", m.group(1))]

    m = re.search(r"answer=\{(\d+)\}", body)
    answer = int(m.group(1)) if m else -1

    out = [f"**Q. {question}**", ""]
    for i, choice in enumerate(choices):
        out.append(f"  {i + 1}. {choice}")
    out.append("")
    if 0 <= answer < len(choices):
        out.append(f"  → 答え: {answer + 1}. {choices[answer]}")
    if explanation:
        out.append(f"     {explanation}")
    return "\n".join(out) + "\n"


def main() -> None:
    text = sys.stdin.read()

    # importとfrontmatterの余計な行を落とす
    text = re.sub(r"^import .*$", "", text, flags=re.M)

    # <Quiz ... /> を読める形へ
    text = re.sub(r"<Quiz\s+(.*?)/>", render_quiz, text, flags=re.S)

    # <details>/<summary> は折り畳めないので見出しにする
    text = re.sub(r"<summary>(.*?)</summary>", r"> ▼ \1", text, flags=re.S)
    text = re.sub(r"</?details>", "", text)

    # Starlightのアサイド記法を素の引用へ
    text = re.sub(r":::note\[(.*?)\]", r"> **\1**", text)
    text = re.sub(r":::(note|tip|caution|danger)(\[.*?\])?", "> ", text)
    text = re.sub(r"^:::$", "", text, flags=re.M)

    # 空行が続きすぎるのを整理
    text = re.sub(r"\n{4,}", "\n\n\n", text)
    sys.stdout.write(text)


if __name__ == "__main__":
    main()
