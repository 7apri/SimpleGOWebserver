FROM postgres:16

# This installs PostGIS natively on whatever architecture you are on
RUN apt-get update && apt-get install -y \
    postgresql-16-postgis-3 \
    postgis \
    && rm -rf /var/lib/apt/lists/*

COPY --chown=postgres:postgres init-schema.sql /docker-entrypoint-initdb.d/01-init.sql
