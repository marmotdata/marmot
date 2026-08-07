The MongoDB plugin discovers databases and collections from MongoDB instances. It samples documents to infer schema and captures index information.

## Required Permissions

The user needs read access to discover collections:

```javascript
db.createUser({
  user: "marmot_reader",
  pwd: "your-password",
  roles: [{ role: "read", db: "your_database" }]
})
```

For discovering all databases, use the `readAnyDatabase` role.
