# CompleteRunRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**error** | **str** |  | [optional] 
**run_id** | **str** |  | 
**status** | [**RunStatus**](RunStatus.md) |  | 
**summary** | [**RunSummary**](RunSummary.md) |  | [optional] 

## Example

```python
from marmot.generated.models.complete_run_request import CompleteRunRequest

# TODO update the JSON string below
json = "{}"
# create an instance of CompleteRunRequest from a JSON string
complete_run_request_instance = CompleteRunRequest.from_json(json)
# print the JSON string representation of the object
print(CompleteRunRequest.to_json())

# convert the object into a dict
complete_run_request_dict = complete_run_request_instance.to_dict()
# create an instance of CompleteRunRequest from a dict
complete_run_request_from_dict = CompleteRunRequest.from_dict(complete_run_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


