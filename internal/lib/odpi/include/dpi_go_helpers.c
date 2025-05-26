#include "dpi.h"

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