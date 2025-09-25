FROM python:3.11-slim

WORKDIR /usr/app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    git \
    && rm -rf /var/lib/apt/lists/*

# Install dbt with PostgreSQL and OpenLineage
RUN pip install \
    dbt-postgres==1.6.7 \
    openlineage-dbt==1.21.0

# Wait for postgres
RUN pip install psycopg2-binary

COPY wait-for-postgres.py /usr/app/
COPY entrypoint.sh /usr/app/

RUN chmod +x /usr/app/entrypoint.sh

CMD ["/usr/app/entrypoint.sh"]
