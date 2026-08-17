# DestroyRunResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**assets_deleted** | **int** |  | [optional] 
**deleted_entity_mrns** | **List[str]** |  | [optional] 
**documentation_deleted** | **int** |  | [optional] 
**lineage_deleted** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.destroy_run_response import DestroyRunResponse

# TODO update the JSON string below
json = "{}"
# create an instance of DestroyRunResponse from a JSON string
destroy_run_response_instance = DestroyRunResponse.from_json(json)
# print the JSON string representation of the object
print(DestroyRunResponse.to_json())

# convert the object into a dict
destroy_run_response_dict = destroy_run_response_instance.to_dict()
# create an instance of DestroyRunResponse from a dict
destroy_run_response_from_dict = DestroyRunResponse.from_dict(destroy_run_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


