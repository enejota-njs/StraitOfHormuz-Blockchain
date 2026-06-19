FROM python:3.11-alpine

WORKDIR /app

RUN apk add --no-cache build-base sdl2-dev sdl2_image-dev sdl2_mixer-dev sdl2_ttf-dev

COPY ./interface/interface.py ./interface.py

COPY ./data/images ./data/images
COPY ./data/interface ./data/interface

RUN pip install --no-cache-dir pygame

CMD ["python3", "interface.py"]