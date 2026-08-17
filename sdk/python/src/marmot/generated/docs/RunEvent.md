# RunEvent


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**event_time** | **str** |  | [optional] 
**event_type** | **str** |  | [optional] 
**inputs** | [**List[Dataset]**](Dataset.md) |  | [optional] 
**job** | [**Job**](Job.md) |  | [optional] 
**outputs** | [**List[Dataset]**](Dataset.md) |  | [optional] 
**producer** | **str** |  | [optional] 
**run** | [**LineageRun**](LineageRun.md) |  | [optional] 
**schema_url** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.run_event import RunEvent

# TODO update the JSON string below
json = "{}"
# create an instance of RunEvent from a JSON string
run_event_instance = RunEvent.from_json(json)
# print the JSON string representation of the object
print(RunEvent.to_json())

# convert the object into a dict
run_event_dict = run_event_instance.to_dict()
# create an instance of RunEvent from a dict
run_event_from_dict = RunEvent.from_dict(run_event_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


