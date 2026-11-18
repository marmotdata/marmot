#!/usr/bin/env bash
# Runs job.py in local-mode Spark (docker) with openlineage-spark sending events to $OPENLINEAGE_URL.
#
# Env:
#   OPENLINEAGE_URL        required, base URL (the client appends /api/v1/lineage)
#   OPENLINEAGE_API_KEY    optional, sent as a bearer token
#   OPENLINEAGE_NAMESPACE  optional, job namespace (default spark-shop)
#   SPARK_IVY_CACHE        optional, host dir for the ivy cache (default .ivy in this dir)
set -euo pipefail

cd "$(dirname "$0")"

: "${OPENLINEAGE_URL:?OPENLINEAGE_URL must be set}"
NAMESPACE="${OPENLINEAGE_NAMESPACE:-spark-shop}"
IMAGE="apache/spark:4.0.5-scala2.13-java17-python3-ubuntu"
OL_PACKAGE="io.openlineage:openlineage-spark_2.13:1.55.0"
IVY="${SPARK_IVY_CACHE:-$PWD/.ivy}"

# Everything the job reads and writes lives under /data in the container, so dataset names are the same on every machine.
rm -rf work
mkdir -p work/input "$IVY/cache" "$IVY/jars"
cp input/customers.csv work/input/

auth=()
if [[ -n "${OPENLINEAGE_API_KEY:-}" ]]; then
  auth=(--conf spark.openlineage.transport.auth.type=api_key --conf "spark.openlineage.transport.auth.apiKey=${OPENLINEAGE_API_KEY}")
fi

# Write files as the invoking user: under rootless docker that user is root inside the container.
if docker info --format '{{.SecurityOptions}}' | grep -q rootless; then
  run_as="0:0"
else
  run_as="$(id -u):$(id -g)"
fi

docker run --rm --network host \
  --add-host "$(uname -n):127.0.0.1" \
  --user "$run_as" \
  -e HOME=/tmp -e HADOOP_USER_NAME=spark -e SPARK_USER=spark -e SPARK_LOCAL_HOSTNAME=localhost \
  -v "$PWD/job.py:/job/job.py:ro" \
  -v "$PWD/work:/data" \
  -v "$IVY:/ivy" \
  "$IMAGE" \
  /opt/spark/bin/spark-submit \
    --master 'local[1]' \
    --name shop_orders \
    --conf spark.jars.ivy=/ivy \
    --conf "spark.jars.packages=${OL_PACKAGE}" \
    --conf spark.extraListeners=io.openlineage.spark.agent.OpenLineageSparkListener \
    --conf spark.openlineage.transport.type=http \
    --conf "spark.openlineage.transport.url=${OPENLINEAGE_URL}" \
    --conf "spark.openlineage.namespace=${NAMESPACE}" \
    --conf spark.sql.catalogImplementation=in-memory \
    --conf spark.sql.warehouse.dir=/data/warehouse \
    --conf spark.sql.shuffle.partitions=1 \
    --conf spark.ui.enabled=false \
    --conf spark.driver.host=127.0.0.1 \
    "${auth[@]}" \
    /job/job.py
