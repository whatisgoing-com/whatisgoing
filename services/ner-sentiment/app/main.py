from fastapi import FastAPI

app = FastAPI(title="whatisgoing ner-sentiment service")


@app.get("/healthz")
def healthz():
    return {"status": "ok"}
