# CreateRunHistoryRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_mrn** | **str** |  | [optional] 
**event_time** | **str** |  | [optional] 
**event_type** | **str** |  | [optional] 
**job_facets** | **Dict[str, object]** |  | [optional] 
**job_name** | **str** |  | [optional] 
**job_namespace** | **str** |  | [optional] 
**run_facets** | **Dict[str, object]** |  | [optional] 
**run_id** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.create_run_history_request import CreateRunHistoryRequest

# TODO update the JSON string below
json = "{}"
# create an instance of CreateRunHistoryRequest from a JSON string
create_run_history_request_instance = CreateRunHistoryRequest.from_json(json)
# print the JSON string representation of the object
print(CreateRunHistoryRequest.to_json())

# convert the object into a dict
create_run_history_request_dict = create_run_history_request_instance.to_dict()
# create an instance of CreateRunHistoryRequest from a dict
create_run_history_request_from_dict = CreateRunHistoryRequest.from_dict(create_run_history_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


