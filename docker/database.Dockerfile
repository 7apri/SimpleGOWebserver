FROM postgres:16

RUN apt-get update && apt-get install -y \
    postgresql-16-postgis-3 \
    postgis \
    && rm -rf /var/lib/apt/lists/*