# Dataset


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**facets** | **Dict[str, object]** |  | [optional] 
**input_facets** | **Dict[str, object]** |  | [optional] 
**name** | **str** |  | [optional] 
**namespace** | **str** |  | [optional] 
**output_facets** | **Dict[str, object]** |  | [optional] 

## Example

```python
from marmot.generated.models.dataset import Dataset

# TODO update the JSON string below
json = "{}"
# create an instance of Dataset from a JSON string
dataset_instance = Dataset.from_json(json)
# print the JSON string representation of the object
print(Dataset.to_json())

# convert the object into a dict
dataset_dict = dataset_instance.to_dict()
# create an instance of Dataset from a dict
dataset_from_dict = Dataset.from_dict(dataset_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


