#include "dpi.h"
#include "stddef.h"

/**
 * @brief Gets byte array pointer from dpiData
 * @param data pointer to the dpiData structure
 * @return Pointer to the byte array
 */
const char* getAsBytesPtr(dpiData* data) {
	return (const char*)data->value.asBytes.ptr;
}

/**
 * @brief Gets the length of the byte array from dpiData
 * @param data pointer to the dpiData structure
 * @return Length of the byte array as uint32_t
 */
uint32_t getAsBytesLength(dpiData* data) {
	return data->value.asBytes.length;
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