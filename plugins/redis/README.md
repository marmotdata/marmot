The Redis plugin discovers logical databases (db0–db15) from Redis instances. It uses the `INFO` command to collect server metadata and parses the Keyspace section to identify databases that contain keys.
A database is identified in its MRN by its number alone (`db0`), because a Redis server has no name of its own. Set `zone` to a short name such as `sessions` for each server so that two servers do not share the same `db0` asset. The name is one you choose, not a cloud availability zone.


## Required Permissions

The connecting user needs permission to run the `INFO` command. By default all users can run `INFO`, but if you are using Redis ACLs:

```
ACL SETUSER marmot_reader on >password ~* &* +info +ping +select
```
