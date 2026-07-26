#include <stdio.h>

int main(void) {
    char name[128] = {0};
    if (fgets(name, sizeof(name), stdin) == NULL) {
        return 1;
    }
    for (char *p = name; *p != '\0'; p++) {
        if (*p == '\n' || *p == '\r') {
            *p = '\0';
            break;
        }
    }
    printf("hello from c: %s\n", name);
    return 0;
}
