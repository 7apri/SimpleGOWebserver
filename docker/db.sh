#!/bin/bash
docker compose exec database psql -d db -U dev
