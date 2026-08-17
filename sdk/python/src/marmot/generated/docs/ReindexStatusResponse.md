# ReindexStatusResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**es_configured** | **bool** |  | [optional] 
**running** | **bool** |  | [optional] 

## Example

```python
from marmot.generated.models.reindex_status_response import ReindexStatusResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ReindexStatusResponse from a JSON string
reindex_status_response_instance = ReindexStatusResponse.from_json(json)
# print the JSON string representation of the object
print(ReindexStatusResponse.to_json())

# convert the object into a dict
reindex_status_response_dict = reindex_status_response_instance.to_dict()
# create an instance of ReindexStatusResponse from a dict
reindex_status_response_from_dict = ReindexStatusResponse.from_dict(reindex_status_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


