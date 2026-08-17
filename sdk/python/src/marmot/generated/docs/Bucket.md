# Bucket


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**error** | **int** |  | [optional] 
**hour** | **str** |  | [optional] 
**success** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.bucket import Bucket

# TODO update the JSON string below
json = "{}"
# create an instance of Bucket from a JSON string
bucket_instance = Bucket.from_json(json)
# print the JSON string representation of the object
print(Bucket.to_json())

# convert the object into a dict
bucket_dict = bucket_instance.to_dict()
# create an instance of Bucket from a dict
bucket_from_dict = Bucket.from_dict(bucket_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


