#include <stdlib.h>
#include <stdint.h>
#include <stdio.h>
#include <stddef.h>
#include <string.h>

#include "dpi.h"

// Windows compatibility: Map strncasecmp to _strnicmp for MSVC
#if defined(_WIN32) && !defined(__MINGW32__)
#define strncasecmp _strnicmp
#endif

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
 * getLobFromData - Extract LOB from dpiData
 */
dpiLob* getLobFromData(dpiData* data) {
    if (data == NULL || data->isNull) return NULL;
    return data->value.asLOB;
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
 * @param typeName: Fully qualified type name (e.g., "OMNI_TRACER_API.CLOB_TAB")
 * @param objType: Output parameter for the object type
 * @return: DPI_SUCCESS or DPI_FAILURE
 */
int createCollectionType(dpiConn *conn, const char* typeName, dpiObjectType **objType) {
    if (conn == NULL || typeName == NULL || objType == NULL) {
        return DPI_FAILURE;
    }
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
    if (objType == NULL || obj == NULL) {
        return DPI_FAILURE;
    }
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
    if (obj == NULL || size == NULL) {
        return DPI_FAILURE;
    }
    return dpiObject_getSize(obj, size);
}

/**
 * getObjectType - Gets the object type for a given schema and type name
 * 
 * @param conn: Database connection
 * @param schema: Schema name (usually your username)
 * @param typeName: Type name (e.g., "CLOB_TAB")
 * @param objType: Output parameter for the object type
 * @return: DPI_SUCCESS or DPI_FAILURE
 */
int getObjectType(dpiConn* conn, const char* schema, const char* typeName, dpiObjectType** objType) {
    if (conn == NULL || schema == NULL || typeName == NULL || objType == NULL) {
        return DPI_FAILURE;
    }

    // Construct fully qualified type name
    size_t schemaLen = strlen(schema);
    size_t typeNameLen = strlen(typeName);
    size_t fullTypeNameLen = schemaLen + 1 + typeNameLen; // for "."

    char* fullTypeName = (char*)malloc(fullTypeNameLen + 1); // +1 for null terminator
    if (fullTypeName == NULL) { 
        return DPI_FAILURE; 
    }

    // Build the full type name
    snprintf(fullTypeName, fullTypeNameLen + 1, "%s.%s", schema, typeName);

    // Get the object type from oracle
    int status = dpiConn_getObjectType(conn, fullTypeName, (uint32_t)fullTypeNameLen, objType);
    free(fullTypeName);
    return status;
}

/**
 * getObjectAttributeByName - Gets the attribute type of an object by attribute name
 * 
 * @param objType: The object type
 * @param attrName: The attribute name to look for
 * @param attrType: Output parameter for the attribute type
 * @return: DPI_SUCCESS or DPI_FAILURE
 */
int getObjectAttributeByName(dpiObjectType* objType, const char* attrName, dpiObjectAttr** attrType) {
    if (objType == NULL || attrName == NULL || attrType == NULL) {
        return DPI_FAILURE;
    }

    dpiObjectTypeInfo typeInfo;
    if (dpiObjectType_getInfo(objType, &typeInfo) != DPI_SUCCESS) {
        return DPI_FAILURE;
    }

    // Allocate array for all attributes
    dpiObjectAttr** attributes = (dpiObjectAttr**)malloc(typeInfo.numAttributes * sizeof(dpiObjectAttr*));
    if (attributes == NULL) {
        return DPI_FAILURE;
    }

    // Get all attributes at once
    if (dpiObjectType_getAttributes(objType, typeInfo.numAttributes, attributes) != DPI_SUCCESS) {
        free(attributes);
        return DPI_FAILURE;
    }

    int result = DPI_FAILURE;
    size_t attrNameLen = strlen(attrName);

    for (uint16_t i = 0; i < typeInfo.numAttributes; i++) {
        dpiObjectAttrInfo attrInfo;
        if (dpiObjectAttr_getInfo(attributes[i], &attrInfo) != DPI_SUCCESS) {
            continue;
        }

        // compare attribute names (case-insensitive)
        if (attrNameLen == attrInfo.nameLength && 
            strncasecmp(attrInfo.name, attrName, attrInfo.nameLength) == 0) {
            // Found the attribute - add reference before returning
            if (dpiObjectAttr_addRef(attributes[i]) == DPI_SUCCESS) {
                *attrType = attributes[i];
                result = DPI_SUCCESS;
            }
            break;
        }
    }

    // Release all attributes obtained from getAttributes
    for (uint16_t i = 0; i < typeInfo.numAttributes; i++) {
        dpiObjectAttr_release(attributes[i]);
    }
    free(attributes);

    // return failure if attribute not found
    return result; 
}