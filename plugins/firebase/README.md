The Firebase plugin discovers Firestore databases with their collections and subcollections, and Realtime Database instances, from a Firebase project.

Firestore stores no schema, so each collection's fields are inferred from a sample of its documents. A field missing from any sampled document, or null in one, is nullable; a field seen with two different types is `mixed`.

A subcollection exists once per parent document, so the copies are catalogued as one asset with the document id left out of the name: `firestore/(default)/orders/lines`.

Cloud Storage for Firebase buckets are not discovered here. They are GCS buckets and the `gcs` plugin already owns that identity.

## Databases

Leaving `databases` empty lists every Firestore database through the admin API. The Firestore emulator does not implement that call, so name the databases explicitly when running against one.

## Testing against the emulator

```
docker run -d --name firestore-emulator -p 8080:8080 \
  gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
  gcloud emulators firestore start --host-port=0.0.0.0:8080

MARMOT_TEST_FIRESTORE_EMULATOR_HOST=127.0.0.1:8080 go test ./...
```
