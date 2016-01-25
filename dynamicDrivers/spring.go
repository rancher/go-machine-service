package dynamicDrivers

import (
	"bytes"
	"text/template"
)

const bodyTemplate = `{{range .}}                <value>{{print .}}Config,parent=baseMachineConfig</value>  
{{end}}`

const header = `<beans xmlns="http://www.springframework.org/schema/beans"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xmlns:context="http://www.springframework.org/schema/context"
    xmlns:aop="http://www.springframework.org/schema/aop"
    xmlns:util="http://www.springframework.org/schema/util"
    xsi:schemaLocation="http://www.springframework.org/schema/aop http://www.springframework.org/schema/aop/spring-aop-3.0.xsd
		http://www.springframework.org/schema/beans http://www.springframework.org/schema/beans/spring-beans-3.0.xsd
		http://www.springframework.org/schema/util http://www.springframework.org/schema/util/spring-util-3.2.xsd
		http://www.springframework.org/schema/context http://www.springframework.org/schema/context/spring-context-3.0.xsd">

    <bean id="MachineAddonTypeSet" class="io.cattle.platform.object.meta.TypeSet" >
        <property name="typeClasses" >
            <set>
                <value>io.cattle.platform.docker.machine.api.addon.BaseMachineConfig</value> 
            </set>
        </property>
        <property name="typeNames">
            <set>
`

const footer = `            </set>
        </property>
    </bean>

    <bean id="MachineTypeSet" class="io.cattle.platform.object.meta.TypeSet" >
        <property name="typeNames">
            <set>
                <value>machine,parent=physicalHost</value>
            </set>
        </property>
    </bean>

    <bean class="io.cattle.platform.docker.machine.api.filter.MachineValidationFilter" />
    <bean class="io.cattle.platform.docker.machine.api.MachineLinkFilter" />
</beans>
`

func generateBody(driverConfigs []string) string {
	t := template.New("xmlBody")
	t.Parse(bodyTemplate)
	var s bytes.Buffer
	t.Execute(&s, driverConfigs)
	return s.String()
}

func GenerateSpringContext(resourceData *ResourceData) error {
	toWrite := header + generateBody(resourceData.Drivers) + footer
	return writeToFile([]byte(toWrite), "spring-docker-machine-api-context.xml")
}
