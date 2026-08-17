# AssetsWithSchemasResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**count** | **int** |  | [optional] 
**percentage** | **float** |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.assets_with_schemas_response import AssetsWithSchemasResponse

# TODO update the JSON string below
json = "{}"
# create an instance of AssetsWithSchemasResponse from a JSON string
assets_with_schemas_response_instance = AssetsWithSchemasResponse.from_json(json)
# print the JSON string representation of the object
print(AssetsWithSchemasResponse.to_json())

# convert the object into a dict
assets_with_schemas_response_dict = assets_with_schemas_response_instance.to_dict()
# create an instance of AssetsWithSchemasResponse from a dict
assets_with_schemas_response_from_dict = AssetsWithSchemasResponse.from_dict(assets_with_schemas_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


