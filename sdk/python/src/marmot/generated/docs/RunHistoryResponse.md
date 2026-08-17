# RunHistoryResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**limit** | **int** |  | [optional] 
**offset** | **int** |  | [optional] 
**run_history** | [**List[RunHistory]**](RunHistory.md) |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.run_history_response import RunHistoryResponse

# TODO update the JSON string below
json = "{}"
# create an instance of RunHistoryResponse from a JSON string
run_history_response_instance = RunHistoryResponse.from_json(json)
# print the JSON string representation of the object
print(RunHistoryResponse.to_json())

# convert the object into a dict
run_history_response_dict = run_history_response_instance.to_dict()
# create an instance of RunHistoryResponse from a dict
run_history_response_from_dict = RunHistoryResponse.from_dict(run_history_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


