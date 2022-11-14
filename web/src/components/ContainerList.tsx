import ContainerEntry from "../models/ContainerEntry";
import ContainerListEntry from "./ContainerListEntry";

interface ContainerListProps {
    containers: ContainerEntry[];
    onContainerClick: (container: ContainerEntry) => void;
}

const ContainerList = (props: ContainerListProps) => (
    <ul className="list-group">
        {props.containers.map(c => <ContainerListEntry {...c} onClick={() => props.onContainerClick(c)} />)}
    </ul >
);

export default ContainerList;