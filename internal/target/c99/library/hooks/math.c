// Hooks of math, which Go implements in assembly (on some architectures).
#include <go/math.h>
#include <math.h>

#define S(name) name##_go_math_package

go_f8 S(archMax)(go_f8 x, go_f8 y) { // see math.max.
    if (isinf(x) && x > 0) return x;
    if (isinf(y) && y > 0) return y;
    if (isnan(x) || isnan(y)) return NAN;
    if (x == 0 && x == y) return signbit(x) ? y : x;
    return x > y ? x : y;
}
go_f8 S(archMin)(go_f8 x, go_f8 y) { // see math.min.
    if (isinf(x) && x < 0) return x;
    if (isinf(y) && y < 0) return y;
    if (isnan(x) || isnan(y)) return NAN;
    if (x == 0 && x == y) return signbit(x) ? x : y;
    return x < y ? x : y;
}
go_f8 S(archExp)(go_f8 x) { return exp(x); }
go_f8 S(archFloor)(go_f8 x) { return floor(x); }
go_f8 S(archCeil)(go_f8 x) { return ceil(x); }
go_f8 S(archTrunc)(go_f8 x) { return trunc(x); }
go_f8 S(archHypot)(go_f8 p, go_f8 q) { return hypot(p, q); }
go_f8 S(archLog)(go_f8 x) { return log(x); }
