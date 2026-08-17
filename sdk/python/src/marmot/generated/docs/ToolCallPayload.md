# ToolCallPayload


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**duration_ms** | **int** |  | [optional] 
**started_at** | **str** |  | [optional] 
**status** | **str** |  | [optional] 
**target_mrn** | **str** |  | [optional] 
**tool_name** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.tool_call_payload import ToolCallPayload

# TODO update the JSON string below
json = "{}"
# create an instance of ToolCallPayload from a JSON string
tool_call_payload_instance = ToolCallPayload.from_json(json)
# print the JSON string representation of the object
print(ToolCallPayload.to_json())

# convert the object into a dict
tool_call_payload_dict = tool_call_payload_instance.to_dict()
# create an instance of ToolCallPayload from a dict
tool_call_payload_from_dict = ToolCallPayload.from_dict(tool_call_payload_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


