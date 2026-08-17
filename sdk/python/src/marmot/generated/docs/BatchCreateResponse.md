# BatchCreateResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**assets** | [**List[BatchAssetResult]**](BatchAssetResult.md) |  | [optional] 
**documentation** | [**List[DocumentationResult]**](DocumentationResult.md) |  | [optional] 
**lineage** | [**List[LineageResult]**](LineageResult.md) |  | [optional] 
**stale_entities_removed** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.batch_create_response import BatchCreateResponse

# TODO update the JSON string below
json = "{}"
# create an instance of BatchCreateResponse from a JSON string
batch_create_response_instance = BatchCreateResponse.from_json(json)
# print the JSON string representation of the object
print(BatchCreateResponse.to_json())

# convert the object into a dict
batch_create_response_dict = batch_create_response_instance.to_dict()
# create an instance of BatchCreateResponse from a dict
batch_create_response_from_dict = BatchCreateResponse.from_dict(batch_create_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


