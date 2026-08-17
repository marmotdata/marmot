# BatchDocumentationRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**documentation** | [**List[Documentation]**](Documentation.md) |  | 

## Example

```python
from marmot.generated.models.batch_documentation_request import BatchDocumentationRequest

# TODO update the JSON string below
json = "{}"
# create an instance of BatchDocumentationRequest from a JSON string
batch_documentation_request_instance = BatchDocumentationRequest.from_json(json)
# print the JSON string representation of the object
print(BatchDocumentationRequest.to_json())

# convert the object into a dict
batch_documentation_request_dict = batch_documentation_request_instance.to_dict()
# create an instance of BatchDocumentationRequest from a dict
batch_documentation_request_from_dict = BatchDocumentationRequest.from_dict(batch_documentation_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


