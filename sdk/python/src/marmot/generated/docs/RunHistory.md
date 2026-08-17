# RunHistory


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**duration_ms** | **int** |  | [optional] 
**end_time** | **str** |  | [optional] 
**event_time** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**job_name** | **str** |  | [optional] 
**job_namespace** | **str** |  | [optional] 
**run_id** | **str** |  | [optional] 
**start_time** | **str** |  | [optional] 
**status** | **str** |  | [optional] 
**type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.run_history import RunHistory

# TODO update the JSON string below
json = "{}"
# create an instance of RunHistory from a JSON string
run_history_instance = RunHistory.from_json(json)
# print the JSON string representation of the object
print(RunHistory.to_json())

# convert the object into a dict
run_history_dict = run_history_instance.to_dict()
# create an instance of RunHistory from a dict
run_history_from_dict = RunHistory.from_dict(run_history_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


