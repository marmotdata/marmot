# RunEntity


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**created_at** | **str** |  | [optional] 
**entity_mrn** | **str** |  | [optional] 
**entity_name** | **str** |  | [optional] 
**entity_type** | **str** |  | [optional] 
**error_message** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**run_id** | **str** |  | [optional] 
**status** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.run_entity import RunEntity

# TODO update the JSON string below
json = "{}"
# create an instance of RunEntity from a JSON string
run_entity_instance = RunEntity.from_json(json)
# print the JSON string representation of the object
print(RunEntity.to_json())

# convert the object into a dict
run_entity_dict = run_entity_instance.to_dict()
# create an instance of RunEntity from a dict
run_entity_from_dict = RunEntity.from_dict(run_entity_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


