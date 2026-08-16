from concurrent.futures import ThreadPoolExecutor

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
<p>Critics singled out Mark Zuckerberg's silence on the matter as reckless and dangerous.</p>
</article>
</body>
</html>
"""


def test_extract_returns_typed_entities_with_sentiment():
    resp = client.post("/extract", json={"html": SAMPLE_HTML, "url": "https://example.com/article"})
    assert resp.status_code == 200

    body = resp.json()
    assert body["text"]
    assert body["processing_ms"] > 0

    entities = body["entities"]
    assert entities, "expected at least one entity"

    for ent in entities:
        # ORG is deliberately not extracted right now (2026-08-16) — just
        # PERSON and TOPIC, ORG/others to come back later.
        assert ent["type"] in {"PERSON", "TOPIC"}
        assert -1.0 <= ent["sentiment_score"] <= 1.0
        if ent["type"] != "TOPIC":
            # PERSON is still extractive (GLiNER) — offsets must point
            # back into the returned text. TOPIC is generated, not
            # extracted, so it has no natural span (start/end are 0 by
            # convention, checked separately below).
            assert body["text"][ent["start"] : ent["end"]] == ent["text"]

    musk = next((e for e in entities if e["text"] == "Elon Musk"), None)
    assert musk is not None
    assert musk["type"] == "PERSON"
    assert musk["sentiment_score"] > 0

    zuckerberg = next((e for e in entities if "Zuckerberg" in e["text"]), None)
    assert zuckerberg is not None
    assert zuckerberg["sentiment_score"] < 0

    topics = [e for e in entities if e["type"] == "TOPIC"]
    assert len(topics) == 1, f"expected exactly one generated topic, got {topics!r}"
    assert topics[0]["start"] == 0 and topics[0]["end"] == 0


def test_extract_guarantees_a_topic_for_short_text():
    fragment = "<p>The dog slept.</p>"
    resp = client.post("/extract", json={"html": fragment})
    assert resp.status_code == 200

    topics = [e for e in resp.json()["entities"] if e["type"] == "TOPIC"]
    assert topics, "expected a topic even for text with no named entities"


def test_extract_topic_has_no_leading_article_and_is_short():
    # The topic is generated (issue #37, round 2 — an editor's label for
    # the article, not necessarily a phrase that appears verbatim in it),
    # not extracted. The prompt asks for exactly 2-3 words; in practice a
    # 1.5B model doesn't hit that exactly every time (occasionally produces
    # a single word), so this checks it's short (<=4 words) rather than
    # asserting an exact count the model can't fully guarantee. The
    # no-leading-article instruction has been reliable in testing, so that
    # stays a strict check.
    fragment = "<p>The 55.6-metre concrete figure has stood on the hillside for decades, drawing tourists from across the region.</p>"
    resp = client.post("/extract", json={"html": fragment})
    assert resp.status_code == 200

    topics = [e["text"] for e in resp.json()["entities"] if e["type"] == "TOPIC"]
    assert topics, "expected a topic"
    for topic in topics:
        word_count = len(topic.split())
        assert 1 <= word_count <= 4, f"expected a short topic, got {topic!r} ({word_count} words)"
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
    assert topics, "expected a topic"
    for topic in topics:
        assert topic.lower() not in {"bangladesh", "australia"}, f"bare country name leaked through: {topic!r}"


def test_extract_survives_concurrent_requests():
    # A single Llama instance's context isn't safe for concurrent
    # create_chat_completion calls from multiple threads — this crashed
    # the whole process (SIGSEGV) under real concurrent ingestion load
    # (cmd/core's bounded worker pool), never under sequential requests.
    # Fixed with a lock around the generation call; this locks in that the
    # process survives concurrent load instead of only checking it manually.
    def make_request(i):
        return client.post("/extract", json={"html": f"<p>Article number {i}: a wildfire forced evacuations overnight.</p>"})

    with ThreadPoolExecutor(max_workers=6) as pool:
        responses = list(pool.map(make_request, range(6)))

    for resp in responses:
        assert resp.status_code == 200
        topics = [e for e in resp.json()["entities"] if e["type"] == "TOPIC"]
        assert topics, "expected a topic"


def test_extract_falls_back_for_html_fragment_without_boilerplate():
    fragment = "<p>Apple released a new product and the market reacted well.</p>"
    resp = client.post("/extract", json={"html": fragment})
    assert resp.status_code == 200
    body = resp.json()
    assert "Apple released a new product" in body["text"]


def test_extract_rejects_content_with_no_text():
    resp = client.post("/extract", json={"html": "<html><body></body></html>"})
    assert resp.status_code == 400
