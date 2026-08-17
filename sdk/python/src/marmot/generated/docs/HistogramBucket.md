# HistogramBucket


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**abort** | **int** |  | [optional] 
**complete** | **int** |  | [optional] 
**var_date** | **str** |  | [optional] 
**fail** | **int** |  | [optional] 
**other** | **int** |  | [optional] 
**running** | **int** |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.histogram_bucket import HistogramBucket

# TODO update the JSON string below
json = "{}"
# create an instance of HistogramBucket from a JSON string
histogram_bucket_instance = HistogramBucket.from_json(json)
# print the JSON string representation of the object
print(HistogramBucket.to_json())

# convert the object into a dict
histogram_bucket_dict = histogram_bucket_instance.to_dict()
# create an instance of HistogramBucket from a dict
histogram_bucket_from_dict = HistogramBucket.from_dict(histogram_bucket_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


