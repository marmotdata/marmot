# RunEntitiesResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**entities** | [**List[RunEntity]**](RunEntity.md) |  | [optional] 
**limit** | **int** |  | [optional] 
**offset** | **int** |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.run_entities_response import RunEntitiesResponse

# TODO update the JSON string below
json = "{}"
# create an instance of RunEntitiesResponse from a JSON string
run_entities_response_instance = RunEntitiesResponse.from_json(json)
# print the JSON string representation of the object
print(RunEntitiesResponse.to_json())

# convert the object into a dict
run_entities_response_dict = run_entities_response_instance.to_dict()
# create an instance of RunEntitiesResponse from a dict
run_entities_response_from_dict = RunEntitiesResponse.from_dict(run_entities_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


