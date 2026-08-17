# RunsGet200Response


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**limit** | **int** |  | [optional] 
**offset** | **int** |  | [optional] 
**pipelines** | **List[str]** |  | [optional] 
**runs** | [**List[PluginRun]**](PluginRun.md) |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.runs_get200_response import RunsGet200Response

# TODO update the JSON string below
json = "{}"
# create an instance of RunsGet200Response from a JSON string
runs_get200_response_instance = RunsGet200Response.from_json(json)
# print the JSON string representation of the object
print(RunsGet200Response.to_json())

# convert the object into a dict
runs_get200_response_dict = runs_get200_response_instance.to_dict()
# create an instance of RunsGet200Response from a dict
runs_get200_response_from_dict = RunsGet200Response.from_dict(runs_get200_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


