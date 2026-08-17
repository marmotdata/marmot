# DocumentationCreateRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**content** | **str** |  | 
**mrn** | **str** |  | 
**source** | **str** |  | 

## Example

```python
from marmot.generated.models.documentation_create_request import DocumentationCreateRequest

# TODO update the JSON string below
json = "{}"
# create an instance of DocumentationCreateRequest from a JSON string
documentation_create_request_instance = DocumentationCreateRequest.from_json(json)
# print the JSON string representation of the object
print(DocumentationCreateRequest.to_json())

# convert the object into a dict
documentation_create_request_dict = documentation_create_request_instance.to_dict()
# create an instance of DocumentationCreateRequest from a dict
documentation_create_request_from_dict = DocumentationCreateRequest.from_dict(documentation_create_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


