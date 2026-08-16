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


def test_extract_falls_back_for_html_fragment_without_boilerplate():
    fragment = "<p>Apple released a new product and the market reacted well.</p>"
    resp = client.post("/extract", json={"html": fragment})
    assert resp.status_code == 200
    body = resp.json()
    assert "Apple released a new product" in body["text"]


def test_extract_rejects_content_with_no_text():
    resp = client.post("/extract", json={"html": "<html><body></body></html>"})
    assert resp.status_code == 400
