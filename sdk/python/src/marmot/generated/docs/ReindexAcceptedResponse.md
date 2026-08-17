# ReindexAcceptedResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**message** | **str** |  | [optional] 
**status** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.reindex_accepted_response import ReindexAcceptedResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ReindexAcceptedResponse from a JSON string
reindex_accepted_response_instance = ReindexAcceptedResponse.from_json(json)
# print the JSON string representation of the object
print(ReindexAcceptedResponse.to_json())

# convert the object into a dict
reindex_accepted_response_dict = reindex_accepted_response_instance.to_dict()
# create an instance of ReindexAcceptedResponse from a dict
reindex_accepted_response_from_dict = ReindexAcceptedResponse.from_dict(reindex_accepted_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


