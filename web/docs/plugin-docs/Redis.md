The Redis plugin discovers logical databases (db0–db15) from Redis instances. It uses the `INFO` command to collect server metadata and parses the Keyspace section to identify databases that contain keys.

## Required Permissions

The connecting user needs permission to run the `INFO` command. By default all users can run `INFO`, but if you are using Redis ACLs:

```
ACL SETUSER marmot_reader on >password ~* &* +info +ping +select
```
