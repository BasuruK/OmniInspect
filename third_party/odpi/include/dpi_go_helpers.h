#ifndef DPI_GO_HELPERS_H
#define DPI_GO_HELPERS_H

#include "dpi.h"

const char* getAsBytesPtr(dpiData* data); // returns a pointer to the data as bytes
uint32_t getAsBytesLength(dpiData* data); // returns the length of the data as bytes

//TODO: remove unused converion functions for int64, uint64, double, float

int64_t getAsInt64(dpiData* data); // returns the data as int64
uint64_t getAsUint64(dpiData* data); // returns the data as uint64
double getAsDouble(dpiData* data); // returns the data as double
float getAsFloat(dpiData* data); // returns the data as float

void initDPIDataAsBytes(dpiData* data, const char* ptr, uint32_t length); // initializes dpiData as bytes
void initDPIDataAsInt64(dpiData* data, int64_t value); // initializes dpiData as int64
void initDPIDataAsDouble(dpiData* data, double value);  // initializes dpiData as double

#endif