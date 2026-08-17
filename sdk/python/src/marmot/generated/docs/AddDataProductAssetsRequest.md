# AddDataProductAssetsRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_ids** | **List[str]** |  | 

## Example

```python
from marmot.generated.models.add_data_product_assets_request import AddDataProductAssetsRequest

# TODO update the JSON string below
json = "{}"
# create an instance of AddDataProductAssetsRequest from a JSON string
add_data_product_assets_request_instance = AddDataProductAssetsRequest.from_json(json)
# print the JSON string representation of the object
print(AddDataProductAssetsRequest.to_json())

# convert the object into a dict
add_data_product_assets_request_dict = add_data_product_assets_request_instance.to_dict()
# create an instance of AddDataProductAssetsRequest from a dict
add_data_product_assets_request_from_dict = AddDataProductAssetsRequest.from_dict(add_data_product_assets_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


