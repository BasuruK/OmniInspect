#include <stddef.h>
#include <stdint.h>
#include <string.h> 
#include "dpi.h"

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

/**
 * @brief Initialize dpiData as object (for PL/SQL collections and objects)
 * @param data pointer to the dpiData structure
 * @param obj pointer to the dpiObject
 */
void initDPIDataAsObject(dpiData* data, dpiObject* obj) {
    if (data == NULL) return;
    data->isNull = (obj == NULL) ? 1 : 0;
    data->value.asObject = obj;
}

/**
 * createCollectionType - Gets the object type for a PL/SQL collection
 * 
 * @param conn: Database connection
 * @param schema: Schema name (usually your username)
 * @param typeName: Fully qualified type name (e.g., "OMNI_TRACER_API.CLOB_TAB")
 * @param objType: Output parameter for the object type
 * @return: DPI_SUCCESS or DPI_FAILURE
 */
int createCollectionType(dpiConn *conn, const char* typeName, dpiObjectType **objType) {
    return dpiConn_getObjectType(conn, typeName, (uint32_t)strlen(typeName), objType);
}

/**
 * createCollection - Creates a new collection object instance
 * 
 * @param objType: The collection type
 * @param obj: Output parameter for the created object
 * @return: DPI_SUCCESS or DPI_FAILURE
 */
int createCollection(dpiObjectType* objType, dpiObject** obj) {
    return dpiObjectType_createObject(objType, obj);
}

/**
 * getCollectionSize - Gets the number of elements in a collection
 * 
 * @param obj: The collection object
 * @param size: Output parameter for the size
 * @return: DPI_SUCCESS or DPI_FAILURE
 */
int getCollectionSize(dpiObject* obj, int32_t* size) {
    return dpiObject_getSize(obj, size);
}

/**
 * getCollectionElementAsString - Gets an element from a CLOB collection as string
 * 
 * @param obj: The collection object
 * @param index: Element index (1-based, like PL/SQL)
 * @param value: Output buffer for the string
 * @param valueLen: Output parameter for string length
 * @return: DPI_SUCCESS or DPI_FAILURE
 */
int getCollectionElementAsString(dpiObject* obj, int32_t index, char** value, uint32_t* valueLen) {
    dpiData data;
    
    if (dpiObject_getElementValueByIndex(obj, index, DPI_NATIVE_TYPE_BYTES, &data) != DPI_SUCCESS) {
        return DPI_FAILURE;
    }
    
    *value = (char*)data.value.asBytes.ptr;
    *valueLen = data.value.asBytes.length;
    return DPI_SUCCESS;
}

/**
 * getCollectionElementAsRaw - Gets an element from a RAW collection
 * 
 * @param obj: The collection object
 * @param index: Element index (1-based)
 * @param value: Output buffer for raw bytes
 * @param valueLen: Output parameter for byte length
 * @return: DPI_SUCCESS or DPI_FAILURE
 */
int getCollectionElementAsRaw(dpiObject* obj, int32_t index, const char** value, uint32_t* valueLen) {
    dpiData data;
    
    if (dpiObject_getElementValueByIndex(obj, index, DPI_NATIVE_TYPE_BYTES, &data) != DPI_SUCCESS) {
        return DPI_FAILURE;
    }
    
    *value = (const char*)data.value.asBytes.ptr;
    *valueLen = data.value.asBytes.length;
    return DPI_SUCCESS;
}