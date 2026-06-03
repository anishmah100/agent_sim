FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY my_bot.py .
CMD ["python", "-u", "my_bot.py"]
