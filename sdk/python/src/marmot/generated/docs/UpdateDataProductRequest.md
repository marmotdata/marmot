# UpdateDataProductRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**description** | **str** |  | [optional] 
**metadata** | **Dict[str, object]** |  | [optional] 
**name** | **str** |  | [optional] 
**owners** | [**List[DataProductOwnerRequest]**](DataProductOwnerRequest.md) |  | [optional] 
**tags** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.update_data_product_request import UpdateDataProductRequest

# TODO update the JSON string below
json = "{}"
# create an instance of UpdateDataProductRequest from a JSON string
update_data_product_request_instance = UpdateDataProductRequest.from_json(json)
# print the JSON string representation of the object
print(UpdateDataProductRequest.to_json())

# convert the object into a dict
update_data_product_request_dict = update_data_product_request_instance.to_dict()
# create an instance of UpdateDataProductRequest from a dict
update_data_product_request_from_dict = UpdateDataProductRequest.from_dict(update_data_product_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


