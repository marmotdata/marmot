# UpdateServiceAccountRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**active** | **bool** |  | [optional] 
**description** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**role_ids** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.update_service_account_request import UpdateServiceAccountRequest

# TODO update the JSON string below
json = "{}"
# create an instance of UpdateServiceAccountRequest from a JSON string
update_service_account_request_instance = UpdateServiceAccountRequest.from_json(json)
# print the JSON string representation of the object
print(UpdateServiceAccountRequest.to_json())

# convert the object into a dict
update_service_account_request_dict = update_service_account_request_instance.to_dict()
# create an instance of UpdateServiceAccountRequest from a dict
update_service_account_request_from_dict = UpdateServiceAccountRequest.from_dict(update_service_account_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


