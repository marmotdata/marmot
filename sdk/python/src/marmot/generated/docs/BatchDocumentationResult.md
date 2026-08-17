# BatchDocumentationResult


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**documentation** | [**Documentation**](Documentation.md) |  | [optional] 
**error** | **str** |  | [optional] 
**status** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.batch_documentation_result import BatchDocumentationResult

# TODO update the JSON string below
json = "{}"
# create an instance of BatchDocumentationResult from a JSON string
batch_documentation_result_instance = BatchDocumentationResult.from_json(json)
# print the JSON string representation of the object
print(BatchDocumentationResult.to_json())

# convert the object into a dict
batch_documentation_result_dict = batch_documentation_result_instance.to_dict()
# create an instance of BatchDocumentationResult from a dict
batch_documentation_result_from_dict = BatchDocumentationResult.from_dict(batch_documentation_result_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


