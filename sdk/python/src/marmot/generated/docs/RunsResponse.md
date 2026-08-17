# RunsResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**runs** | [**List[AgentRun]**](AgentRun.md) |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.runs_response import RunsResponse

# TODO update the JSON string below
json = "{}"
# create an instance of RunsResponse from a JSON string
runs_response_instance = RunsResponse.from_json(json)
# print the JSON string representation of the object
print(RunsResponse.to_json())

# convert the object into a dict
runs_response_dict = runs_response_instance.to_dict()
# create an instance of RunsResponse from a dict
runs_response_from_dict = RunsResponse.from_dict(runs_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


