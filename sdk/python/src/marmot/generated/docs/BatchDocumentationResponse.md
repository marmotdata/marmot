# BatchDocumentationResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**results** | [**List[BatchDocumentationResult]**](BatchDocumentationResult.md) |  | [optional] 

## Example

```python
from marmot.generated.models.batch_documentation_response import BatchDocumentationResponse

# TODO update the JSON string below
json = "{}"
# create an instance of BatchDocumentationResponse from a JSON string
batch_documentation_response_instance = BatchDocumentationResponse.from_json(json)
# print the JSON string representation of the object
print(BatchDocumentationResponse.to_json())

# convert the object into a dict
batch_documentation_response_dict = batch_documentation_response_instance.to_dict()
# create an instance of BatchDocumentationResponse from a dict
batch_documentation_response_from_dict = BatchDocumentationResponse.from_dict(batch_documentation_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


