# CreateDocRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_mrn** | **str** |  | [optional] 
**content** | **str** |  | [optional] 
**type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.create_doc_request import CreateDocRequest

# TODO update the JSON string below
json = "{}"
# create an instance of CreateDocRequest from a JSON string
create_doc_request_instance = CreateDocRequest.from_json(json)
# print the JSON string representation of the object
print(CreateDocRequest.to_json())

# convert the object into a dict
create_doc_request_dict = create_doc_request_instance.to_dict()
# create an instance of CreateDocRequest from a dict
create_doc_request_from_dict = CreateDocRequest.from_dict(create_doc_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


