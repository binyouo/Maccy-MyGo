#include <libproc.h>
#include <sys/resource.h>
#include <mach/mach_time.h>
#include <unistd.h>
#include <stdio.h>
int main(void) {
 struct mach_timebase_info t; mach_timebase_info(&t);
 struct rusage r; struct rusage_info_v2 info;
 volatile double n=0; uint64_t end=mach_absolute_time()+(uint64_t)(200000000.0*t.denom/t.numer);
 while(mach_absolute_time()<end)n+=1;
 getrusage(RUSAGE_SELF,&r);proc_pid_rusage(getpid(),RUSAGE_INFO_V2,(rusage_info_t*)&info);
 printf("timebase=%u/%u getrusage_user_s=%.6f ri_user_time_raw=%llu ri_user_as_ticks_s=%.6f\n",t.numer,t.denom,r.ru_utime.tv_sec+r.ru_utime.tv_usec/1000000.,info.ri_user_time,info.ri_user_time*(double)t.numer/t.denom/1e9);
}
