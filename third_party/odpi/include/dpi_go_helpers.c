#include "dpi.h"
#include "stddef.h"

/**
 * @brief Gets byte array pointer from dpiData
 * @param data pointer to the dpiData structure
 * @return Pointer to the byte array
 */
const char* getAsBytesPtr(dpiData* data) {
    if (data == NULL || data->isNull) return NULL;
	return (const char*)data->value.asBytes.ptr;
}

/**
 * @brief Gets the length of the byte array from dpiData
 * @param data pointer to the dpiData structure
 * @return Length of the byte array as uint32_t
 */
uint32_t getAsBytesLength(dpiData* data) {
    if (data == NULL || data->isNull) return 0;
	return data->value.asBytes.length;
}

/**
 * @brief Gets int64 value from dpiData
 * @param data pointer to the dpiData structure
 * @return The int64 value
 */
int64_t getAsInt64(dpiData* data) {
    if (data == NULL || data->isNull) return 0;
	return data->value.asInt64;
}

/**
 * @brief Gets uint64 value from dpiData
 * @param data pointer to the dpiData structure
 * @return The uint64 value
 */
uint64_t getAsUint64(dpiData* data) {
    if (data == NULL || data->isNull) return 0;
	return data->value.asUint64;
}

/**
 * @brief Gets double value from dpiData
 * @param data pointer to the dpiData structure
 * @return The double value
 */
double getAsDouble(dpiData* data) {
    if (data == NULL || data->isNull) return 0;
	return data->value.asDouble;
}

/**
 * @brief Gets float value from dpiData
 * @param data pointer to the dpiData structure
 * @return The float value
 */
float getAsFloat(dpiData* data) {
    if (data == NULL || data->isNull) return 0;
	return data->value.asFloat;
}

/**
 * @brief Initialize dpiData as bytes
 * @param data pointer to the dpiData structure
 * @param ptr pointer to the byte data
 * @param length length of the byte data
 */
void initDPIDataAsBytes(dpiData* data, const char* ptr, uint32_t length) {
    if (data == NULL) return;
    data->isNull = 0;
    data->value.asBytes.ptr = (char*)ptr;
    data->value.asBytes.length = length;
}

/**
 * @brief Initialize dpiData as int64
 * @param data pointer to the dpiData structure
 * @param value the int64 value
 */
void initDPIDataAsInt64(dpiData* data, int64_t value) {
    if (data == NULL) return;
    data->isNull = 0;
    data->value.asInt64 = value;
}

/**
 * @brief Initialize dpiData as double
 * @param data pointer to the dpiData structure
 * @param value the double value
 */
void initDPIDataAsDouble(dpiData* data, double value) {
    if (data == NULL) return;
    data->isNull = 0;
    data->value.asDouble = value;
}

/**
 * @brief Initialize dpiData as uint64
 * @param data pointer to the dpiData structure
 * @param value the uint64 value
 */
void initDPIDataAsUint64(dpiData* data, uint64_t value) {
    if (data == NULL) return;
    data->isNull = 0;
    data->value.asUint64 = value;
}

/**
 * @brief Initialize dpiData as float
 * @param data pointer to the dpiData structure
 * @param value the float value
 */
void initDPIDataAsFloat(dpiData* data, float value) {
    if (data == NULL) return;
    data->isNull = 0;
    data->value.asFloat = value;
}