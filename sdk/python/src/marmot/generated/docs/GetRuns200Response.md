# GetRuns200Response


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
from marmot.generated.models.get_runs200_response import GetRuns200Response

# TODO update the JSON string below
json = "{}"
# create an instance of GetRuns200Response from a JSON string
get_runs200_response_instance = GetRuns200Response.from_json(json)
# print the JSON string representation of the object
print(GetRuns200Response.to_json())

# convert the object into a dict
get_runs200_response_dict = get_runs200_response_instance.to_dict()
# create an instance of GetRuns200Response from a dict
get_runs200_response_from_dict = GetRuns200Response.from_dict(get_runs200_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


