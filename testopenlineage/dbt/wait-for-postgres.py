#!/usr/bin/env python3
import psycopg2
import time
import os

def wait_for_postgres():
    max_retries = 30
    retry_interval = 2
    
    for attempt in range(max_retries):
        try:
            conn = psycopg2.connect(
                host="postgres",
                database="dbt_test",
                user="dbt_user",
                password="dbt_password"
            )
            conn.close()
            print("PostgreSQL is ready!")
            return True
        except psycopg2.OperationalError:
            print(f"Waiting for PostgreSQL... (attempt {attempt + 1}/{max_retries})")
            time.sleep(retry_interval)
    
    print("Failed to connect to PostgreSQL")
    return False

if __name__ == "__main__":
    wait_for_postgres()
