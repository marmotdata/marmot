# AgentRun


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**agent_id** | **str** |  | [optional] 
**created_at** | **str** |  | [optional] 
**duration_ms** | **int** |  | [optional] 
**ended_at** | **str** |  | [optional] 
**error** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**model** | **str** |  | [optional] 
**run_id** | **str** |  | [optional] 
**started_at** | **str** |  | [optional] 
**status** | **str** |  | [optional] 
**tokens_in** | **int** |  | [optional] 
**tokens_out** | **int** |  | [optional] 
**tool_calls** | [**List[ToolCall]**](ToolCall.md) |  | [optional] 

## Example

```python
from marmot.generated.models.agent_run import AgentRun

# TODO update the JSON string below
json = "{}"
# create an instance of AgentRun from a JSON string
agent_run_instance = AgentRun.from_json(json)
# print the JSON string representation of the object
print(AgentRun.to_json())

# convert the object into a dict
agent_run_dict = agent_run_instance.to_dict()
# create an instance of AgentRun from a dict
agent_run_from_dict = AgentRun.from_dict(agent_run_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


