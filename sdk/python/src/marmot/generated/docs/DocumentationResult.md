# DocumentationResult


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_mrn** | **str** |  | [optional] 
**error** | **str** |  | [optional] 
**status** | **str** |  | [optional] 
**type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.documentation_result import DocumentationResult

# TODO update the JSON string below
json = "{}"
# create an instance of DocumentationResult from a JSON string
documentation_result_instance = DocumentationResult.from_json(json)
# print the JSON string representation of the object
print(DocumentationResult.to_json())

# convert the object into a dict
documentation_result_dict = documentation_result_instance.to_dict()
# create an instance of DocumentationResult from a dict
documentation_result_from_dict = DocumentationResult.from_dict(documentation_result_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


