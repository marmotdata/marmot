"""Small PySpark scenario for capturing openlineage-spark events.

Reads a shipped CSV and a parquet file it writes itself, joins them, writes parquet, runs a CTAS into a catalog table and then runs one job that fails at execution time.
"""
import sys

from pyspark.sql import SparkSession, functions as F
from pyspark.sql.types import DateType, DoubleType, IntegerType, StringType, StructField, StructType

DATA = "/data"

spark = SparkSession.builder.getOrCreate()
spark.sparkContext.setLogLevel("WARN")

# 1. Generate the orders parquet file that the join reads.
orders_schema = StructType([
    StructField("order_id", IntegerType(), False),
    StructField("customer_id", IntegerType(), False),
    StructField("ordered_at", StringType(), False),
    StructField("status", StringType(), False),
    StructField("amount", DoubleType(), False),
])
orders_rows = [
    (1001, 1, "2026-01-03", "shipped", 42.50),
    (1002, 2, "2026-01-04", "shipped", 18.00),
    (1003, 1, "2026-01-07", "returned", 7.25),
    (1004, 3, "2026-01-09", "placed", 99.99),
    (1005, 4, "2026-01-11", "shipped", 12.00),
]
(spark.createDataFrame(orders_rows, orders_schema)
    .withColumn("ordered_at", F.col("ordered_at").cast(DateType()))
    .coalesce(1)
    .write.mode("overwrite").parquet(f"{DATA}/raw/orders"))

# 2. Join the CSV and the parquet file and write the result as parquet.
customers = (spark.read.option("header", True)
    .schema("customer_id INT, customer_name STRING, country STRING")
    .csv(f"{DATA}/input/customers.csv"))
orders = spark.read.parquet(f"{DATA}/raw/orders")
customer_orders = (customers.join(orders.where(F.col("status") != "returned"), "customer_id", "left")
    .groupBy("customer_id", "customer_name", "country")
    .agg(F.count("order_id").alias("order_count"), F.coalesce(F.sum("amount"), F.lit(0.0)).alias("lifetime_value")))
customer_orders.coalesce(1).write.mode("overwrite").parquet(f"{DATA}/marts/customer_orders")

# 3. CTAS into the in-memory catalog, reading the parquet output back through a temp view.
spark.read.parquet(f"{DATA}/marts/customer_orders").createOrReplaceTempView("customer_orders_v")
spark.sql("CREATE DATABASE IF NOT EXISTS shop")
spark.sql("DROP TABLE IF EXISTS shop.revenue_by_country")
spark.sql("""
    CREATE TABLE shop.revenue_by_country USING parquet AS
    SELECT country, SUM(lifetime_value) AS revenue, SUM(order_count) AS orders
    FROM customer_orders_v
    GROUP BY country
""")
rows = spark.table("shop.revenue_by_country").count()
if rows != 3:
    sys.exit(f"expected 3 countries, got {rows}")

# 4. A job that fails while executing: assert_true raises inside the tasks, so the write starts and then fails.
try:
    (orders.withColumn("check", F.assert_true(F.col("amount") < 0, F.lit("amount must be negative")).cast("string"))
        .write.mode("overwrite").parquet(f"{DATA}/marts/negative_orders"))
except Exception as exc:  # noqa: BLE001
    print(f"failing job failed as expected: {type(exc).__name__}: {str(exc).splitlines()[0]}")
else:
    sys.exit("expected the failing job to fail")

spark.stop()
