# UpdateTermRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**definition** | **str** |  | [optional] 
**description** | **str** |  | [optional] 
**metadata** | **Dict[str, object]** |  | [optional] 
**name** | **str** |  | [optional] 
**owners** | [**List[OwnerRequest]**](OwnerRequest.md) |  | [optional] 
**parent_term_id** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.update_term_request import UpdateTermRequest

# TODO update the JSON string below
json = "{}"
# create an instance of UpdateTermRequest from a JSON string
update_term_request_instance = UpdateTermRequest.from_json(json)
# print the JSON string representation of the object
print(UpdateTermRequest.to_json())

# convert the object into a dict
update_term_request_dict = update_term_request_instance.to_dict()
# create an instance of UpdateTermRequest from a dict
update_term_request_from_dict = UpdateTermRequest.from_dict(update_term_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


