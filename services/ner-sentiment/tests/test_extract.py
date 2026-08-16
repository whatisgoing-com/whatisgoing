from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)


def test_healthz():
    resp = client.get("/healthz")
    assert resp.status_code == 200
    assert resp.json() == {"status": "ok"}


SAMPLE_HTML = """
<html>
<head><title>Sample Article</title></head>
<body>
<article>
<h1>Sample Article</h1>
<p>Elon Musk announced a major breakthrough at Tesla today, delighting investors.</p>
<p>Critics at the United Nations condemned the decision as reckless and dangerous.</p>
</article>
</body>
</html>
"""


def test_extract_returns_typed_offset_accurate_entities_with_sentiment():
    resp = client.post("/extract", json={"html": SAMPLE_HTML, "url": "https://example.com/article"})
    assert resp.status_code == 200

    body = resp.json()
    assert body["text"]
    assert body["processing_ms"] > 0

    entities = body["entities"]
    assert entities, "expected at least one entity"

    for ent in entities:
        assert ent["type"] in {"PERSON", "ORG", "TOPIC"}
        # offsets must point back into the returned text
        assert body["text"][ent["start"] : ent["end"]] == ent["text"]
        assert -1.0 <= ent["sentiment_score"] <= 1.0

    musk = next((e for e in entities if e["text"] == "Elon Musk"), None)
    assert musk is not None
    assert musk["type"] == "PERSON"
    assert musk["sentiment_score"] > 0

    un = next((e for e in entities if "United Nations" in e["text"]), None)
    assert un is not None
    assert un["sentiment_score"] < 0

    topics = [e for e in entities if e["type"] == "TOPIC"]
    assert topics, "expected at least one topic per article"


def test_extract_guarantees_at_least_one_topic_for_short_text():
    fragment = "<p>The dog slept.</p>"
    resp = client.post("/extract", json={"html": fragment})
    assert resp.status_code == 200

    body = resp.json()
    topics = [e for e in body["entities"] if e["type"] == "TOPIC"]
    assert topics, "expected at least one topic even for text with no strong keyphrases"
    for topic in topics:
        assert body["text"][topic["start"] : topic["end"]] == topic["text"]


def test_extract_strips_leading_article_from_fallback_topic():
    # Thin, single-sentence RSS content (common in this dataset) often has
    # no confident GLiNER "topic" candidate, so the longest-noun-chunk
    # fallback kicks in — and raw noun chunks keep a leading article
    # ("The 55.6-metre concrete figure", "a second mass crossing") that
    # reads oddly as a topic label.
    fragment = "<p>The 55.6-metre concrete figure has stood on the hillside for decades.</p>"
    resp = client.post("/extract", json={"html": fragment})
    assert resp.status_code == 200

    topics = [e["text"] for e in resp.json()["entities"] if e["type"] == "TOPIC"]
    assert topics, "expected at least one topic"
    for topic in topics:
        assert not topic.lower().startswith(("the ", "a ", "an ")), f"leading article leaked through: {topic!r}"


def test_extract_excludes_bare_country_names_from_topics():
    fragment = (
        "<p>Bangladesh thrashed Australia in one of Test cricket's greatest "
        "upsets, as the underdogs pulled off a historic quadruple in front "
        "of a stunned crowd.</p>"
    )
    resp = client.post("/extract", json={"html": fragment})
    assert resp.status_code == 200

    topics = [e["text"] for e in resp.json()["entities"] if e["type"] == "TOPIC"]
    assert topics, "expected at least one topic"
    for topic in topics:
        assert topic.lower() not in {"bangladesh", "australia"}, f"bare country name leaked through: {topic!r}"


def test_extract_allows_single_word_topics_when_the_model_is_confident():
    # GLiNER's "topic" label is semantic, unlike the old noun-chunk-length
    # heuristic — a single confident word like "sports" is a real topic,
    # not junk the way a bare noun-chunk word ("court", "company") was.
    # This is a deliberate capability, not an oversight: don't reintroduce
    # a blanket multi-word-only filter without checking this stays true.
    fragment = (
        "<p>Kalshi lets customers bet on the outcome of just about anything: "
        "sports, elections, politics, entertainment, culture, tech, and "
        "science are all covered by its prediction markets, which have "
        "exploded in popularity this year.</p>"
    )
    resp = client.post("/extract", json={"html": fragment})
    assert resp.status_code == 200

    topics = [e["text"] for e in resp.json()["entities"] if e["type"] == "TOPIC"]
    assert any(" " not in topic for topic in topics), f"expected at least one single-word topic, got {topics!r}"


def test_extract_falls_back_for_html_fragment_without_boilerplate():
    fragment = "<p>Apple released a new product and the market reacted well.</p>"
    resp = client.post("/extract", json={"html": fragment})
    assert resp.status_code == 200
    body = resp.json()
    assert "Apple released a new product" in body["text"]


def test_extract_rejects_content_with_no_text():
    resp = client.post("/extract", json={"html": "<html><body></body></html>"})
    assert resp.status_code == 400
