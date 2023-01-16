import { useEffect, useState } from "react";
import ContainerModel from "../models/ContainerModel";
import { check, list, ListResponse, update } from "../services/Api";
import ContainerList from "./ContainerList";
import Spinner from "./Spinner";
import SpinnerModal from "./SpinnerModal";
import { UpdateSelected, UpdateAll, UpdateCheck } from "./UpdateButtons";

const ContainerView = () => {
    const [loading, setLoading] = useState(true);
    const [checking, setChecking] = useState(false);
    const [updating, setUpdating] = useState(false);
    const [updatingImage, setUpdatingContainer] = useState<string | null>(null);
    const [hasChecked, setHasChecked] = useState(false);
    const [containers, setContainers] = useState<ContainerModel[]>([]);

    const containersWithUpdates = containers.filter((c) => c.HasUpdate);
    const containersWithoutUpdates = containers.filter((c) => !c.HasUpdate);
    const selectedContainers = containers.filter((c) => c.Selected);
    const hasUpdates = containersWithUpdates.length > 0;
    const hasSelectedContainers = selectedContainers.length > 0;

    const checkForUpdates = async (containersToUpdate?: ContainerModel[]) => {

        if (!containersToUpdate) {
            containersToUpdate = containers;
        }

        setChecking(true);

        setContainers((current) =>
            current.map((c) => ({
                ...c,
                IsChecking: true
            }))
        );

        await Promise.all(containersToUpdate.map(async (c1) => {
            const result = await check(c1.ContainerID);
            setContainers((current) =>
                current.map((c2) => (c1.ContainerID === c2.ContainerID ? {
                    ...c2,
                    ...result,
                    IsChecking: false
                } : c2
                ))
            );
        }));

        setChecking(false);
        setHasChecked(true);
    };

    const listContainers = async () => {
        setLoading(true);
        const data = await list();
        const mappedData = data.Containers.map((c) => ({
            ...c,
            Selected: false,
            IsChecking: false,
            HasUpdate: false,
            IsUpdating: false,
            NewVersion: "",
            NewVersionCreated: ""
        }));
        setContainers(mappedData);
        setLoading(false);
        setHasChecked(false);
        return mappedData;
    };

    const updateImages = async (containersToUpdate: ContainerModel[]) => {
        setUpdating(true);
        const containerNames = containersToUpdate.map((c) => c.ContainerName);
        for (const containerName of containerNames) {
            setUpdatingContainer(containerName);
            await update([containerName]);
        }
        setUpdatingContainer(null);
        const clist = await listContainers();
        await checkForUpdates(clist);
        setUpdating(false);
    };

    const updateAll = async () => {
        await updateImages(containersWithUpdates);
    };

    const updateSelected = async () => {
        await updateImages(selectedContainers);
    };

    const onContainerClick = (container: ContainerModel) => {
        setContainers((current) =>
            current.map((c2) => (container.ContainerID === c2.ContainerID ? {
                ...c2,
                Selected: !c2.Selected
            } : c2
            ))
        );
    };

    useEffect(() => {
        listContainers();
    }, []);

    return (
        <main className="mt-5 p-5 d-block">
            <SpinnerModal visible={updating} title={`Updating ${updatingImage ?? "containers"}`} message="Please wait..." />
            <div className="row mb-2">
                <div className="col-12 col-md-4 d-flex align-items-center">
                    {hasUpdates
                        ? <span>{containersWithUpdates.length} container{containersWithUpdates.length === 1 ? " has" : "s have"} updates.</span>
                        : checking
                            ? <span>Checking for updates...</span>
                            : (hasChecked && containers.length > 0)
                                ? <><i className="bi bi-check-circle-fill fs-4 text-primary me-2"></i><span>All containers are up to date.</span></>
                                : <span>{containers.length} running container{containers.length !== 1 && "s"} found.</span>}
                </div>
                <div className="col-12 col-md-8 text-end">
                    {hasUpdates && <UpdateSelected onClick={updateSelected} disabled={checking || !hasSelectedContainers} />}
                    {hasUpdates && <UpdateAll onClick={updateAll} disabled={checking} />}
                    <UpdateCheck onClick={() => checkForUpdates()} disabled={checking} />
                </div>
            </div>

            <ContainerList containers={containersWithUpdates} onContainerClick={onContainerClick} />

            {hasUpdates && containersWithoutUpdates.length > 0 &&
                <div className="row mt-4 mb-2">
                    <div className="col-4 d-flex align-items-center">
                        {containersWithoutUpdates.length} container{containersWithoutUpdates.length === 1 ? " is" : "s are"} up to date.
                    </div>
                </div>
            }

            <ContainerList containers={containersWithoutUpdates} onContainerClick={onContainerClick} />

            {loading && <Spinner />}
        </main>
    );
};

export default ContainerView;