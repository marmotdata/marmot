The MLflow plugin discovers registered models, experiments and training datasets from an MLflow tracking server.

Every registered model becomes a Model asset carrying the run behind its newest version: hyperparameters, latest metric values, the experiment it came from and the input features of its signature. Experiments become Experiment assets and the datasets logged to a model's run become Dataset assets, with PRODUCES and FEEDS lineage between them. A dataset read from `s3://` or `gs://` is linked to the bucket asset the S3 or GCS plugin creates.

## Authentication

The tracking server is contacted anonymously unless `username` and `password` (MLflow's basic auth) or `token` (a bearer token, for servers behind a proxy) are set. Set one or the other, not both.

## Model Signatures

Features are read from the run's `mlflow.log-model.history` tag. When the run has none, the plugin reads the MLmodel file of the logged model (MLflow 3, `models:/` sources) or of the run's artifacts, which needs a tracking server that stores or proxies its own artifacts (`--serve-artifacts`). A model whose signature cannot be found is still discovered, without features.
