# CreateTermRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**definition** | **str** |  | 
**description** | **str** |  | [optional] 
**metadata** | **Dict[str, object]** |  | [optional] 
**name** | **str** |  | 
**owners** | [**List[OwnerRequest]**](OwnerRequest.md) |  | [optional] 
**parent_term_id** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.create_term_request import CreateTermRequest

# TODO update the JSON string below
json = "{}"
# create an instance of CreateTermRequest from a JSON string
create_term_request_instance = CreateTermRequest.from_json(json)
# print the JSON string representation of the object
print(CreateTermRequest.to_json())

# convert the object into a dict
create_term_request_dict = create_term_request_instance.to_dict()
# create an instance of CreateTermRequest from a dict
create_term_request_from_dict = CreateTermRequest.from_dict(create_term_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


