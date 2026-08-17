# StartRunRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**config** | **Dict[str, object]** |  | [optional] 
**pipeline_name** | **str** |  | 
**source_name** | **str** |  | 

## Example

```python
from marmot.generated.models.start_run_request import StartRunRequest

# TODO update the JSON string below
json = "{}"
# create an instance of StartRunRequest from a JSON string
start_run_request_instance = StartRunRequest.from_json(json)
# print the JSON string representation of the object
print(StartRunRequest.to_json())

# convert the object into a dict
start_run_request_dict = start_run_request_instance.to_dict()
# create an instance of StartRunRequest from a dict
start_run_request_from_dict = StartRunRequest.from_dict(start_run_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


