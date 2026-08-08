import json
import re
from html.parser import HTMLParser
from typing import Optional, Tuple

import trafilatura


def extract_article(raw_html: str, url: Optional[str]) -> Tuple[Optional[str], str]:
    """Extract (title, plain_text) from raw HTML.

    trafilatura is tuned for full article pages with boilerplate to strip.
    RSS item content is often a short HTML fragment it can't confidently
    extract from, so fall back to a plain tag-strip for those.
    """
    extracted = trafilatura.extract(
        raw_html,
        url=url,
        favor_recall=True,
        include_comments=False,
        include_tables=False,
        output_format="json",
        with_metadata=True,
    )

    if extracted:
        parsed = json.loads(extracted)
        text = (parsed.get("text") or "").strip()
        if text:
            return parsed.get("title") or None, text

    return None, _strip_tags(raw_html).strip()


class _TextExtractor(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self._chunks = []

    def handle_data(self, data: str) -> None:
        self._chunks.append(data)

    def text(self) -> str:
        return re.sub(r"\s+", " ", "".join(self._chunks)).strip()


def _strip_tags(raw_html: str) -> str:
    parser = _TextExtractor()
    parser.feed(raw_html)
    return parser.text()
