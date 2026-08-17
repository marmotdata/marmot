# OwnerRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **str** |  | 
**type** | **str** |  | 

## Example

```python
from marmot.generated.models.owner_request import OwnerRequest

# TODO update the JSON string below
json = "{}"
# create an instance of OwnerRequest from a JSON string
owner_request_instance = OwnerRequest.from_json(json)
# print the JSON string representation of the object
print(OwnerRequest.to_json())

# convert the object into a dict
owner_request_dict = owner_request_instance.to_dict()
# create an instance of OwnerRequest from a dict
owner_request_from_dict = OwnerRequest.from_dict(owner_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


