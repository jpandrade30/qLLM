# Not in the default kustomization (RAM / Kafka). Merge into kustomization.yaml resources if you want them.

## mssql

SQL Server Linux needs `ACCEPT_EULA=Y` and ~2Gi RAM. Add a StatefulSet + source `type: mssql` in `config/qllm.preset.yaml` (new hostEnv). Not wired here so the default sim stays smaller.

## cassandra

CQL + `accessPath.partition` same as Dynamo. Use an official Cassandra image and a CQL init Job, then a `type: cassandra` source. Seed data must not reuse CRM table names.

## ksql

Needs Kafka + ksqlDB. Only pull queries with `accessPath.ksqlKey`. Skip unless you already run that stack.
