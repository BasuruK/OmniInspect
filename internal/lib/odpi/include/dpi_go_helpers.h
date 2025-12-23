#ifndef DPI_GO_HELPERS_H
#define DPI_GO_HELPERS_H

#include "dpi.h"

const char* getAsBytesPtr(dpiData* data); // returns a pointer to the data as bytes
uint32_t getAsBytesLength(dpiData* data); // returns the length of the data as bytes
void initDPIDataAsBytes(dpiData* data, const char* ptr, uint32_t length); // initializes dpiData as bytes
void initDPIDataAsInt64(dpiData* data, int64_t value); // initializes dpiData as int64
void initDPIDataAsDouble(dpiData* data, double value);  // initializes dpiData as double

#endif