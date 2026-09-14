FROM python:3.12-slim
WORKDIR /app
COPY data/input.json scenario.json
COPY scripts/run_showcase.py /app/run_showcase.py
ENTRYPOINT ["python", "/app/run_showcase.py"]

