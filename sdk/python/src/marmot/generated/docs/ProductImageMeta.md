# ProductImageMeta


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**content_type** | **str** |  | [optional] 
**created_at** | **str** |  | [optional] 
**data_product_id** | **str** |  | [optional] 
**filename** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**purpose** | [**ImagePurpose**](ImagePurpose.md) |  | [optional] 
**size_bytes** | **int** |  | [optional] 
**url** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.product_image_meta import ProductImageMeta

# TODO update the JSON string below
json = "{}"
# create an instance of ProductImageMeta from a JSON string
product_image_meta_instance = ProductImageMeta.from_json(json)
# print the JSON string representation of the object
print(ProductImageMeta.to_json())

# convert the object into a dict
product_image_meta_dict = product_image_meta_instance.to_dict()
# create an instance of ProductImageMeta from a dict
product_image_meta_from_dict = ProductImageMeta.from_dict(product_image_meta_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


