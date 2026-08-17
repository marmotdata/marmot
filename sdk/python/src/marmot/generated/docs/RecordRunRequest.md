# RecordRunRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**agent_mrn** | **str** |  | [optional] 
**ended_at** | **str** |  | [optional] 
**error** | **str** |  | [optional] 
**model** | **str** |  | [optional] 
**observed_assets** | **List[str]** |  | [optional] 
**run_id** | **str** |  | [optional] 
**started_at** | **str** |  | [optional] 
**status** | **str** |  | [optional] 
**tokens_in** | **int** |  | [optional] 
**tokens_out** | **int** |  | [optional] 
**tool_calls** | [**List[ToolCallPayload]**](ToolCallPayload.md) |  | [optional] 

## Example

```python
from marmot.generated.models.record_run_request import RecordRunRequest

# TODO update the JSON string below
json = "{}"
# create an instance of RecordRunRequest from a JSON string
record_run_request_instance = RecordRunRequest.from_json(json)
# print the JSON string representation of the object
print(RecordRunRequest.to_json())

# convert the object into a dict
record_run_request_dict = record_run_request_instance.to_dict()
# create an instance of RecordRunRequest from a dict
record_run_request_from_dict = RecordRunRequest.from_dict(record_run_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


