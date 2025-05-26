#ifndef DPI_GO_HELPERS_H
#define DPI_GO_HELPERS_H

#include "dpi.h"

const char* getAsBytesPtr(dpiData* data); // returns a pointer to the data as bytes
uint32_t getAsBytesLength(dpiData* data); // returns the length of the data as bytes

#endif