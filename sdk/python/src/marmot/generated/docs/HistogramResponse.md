# HistogramResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**buckets** | [**List[HistogramBucket]**](HistogramBucket.md) |  | [optional] 
**period** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.histogram_response import HistogramResponse

# TODO update the JSON string below
json = "{}"
# create an instance of HistogramResponse from a JSON string
histogram_response_instance = HistogramResponse.from_json(json)
# print the JSON string representation of the object
print(HistogramResponse.to_json())

# convert the object into a dict
histogram_response_dict = histogram_response_instance.to_dict()
# create an instance of HistogramResponse from a dict
histogram_response_from_dict = HistogramResponse.from_dict(histogram_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


