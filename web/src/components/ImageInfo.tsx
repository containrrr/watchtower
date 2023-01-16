
interface ImageInfoProps {
    version: string;
    created: string;
}

const ImageInfo = (props: ImageInfoProps) => (
    <small className="text-muted mx-2" title={props.version + " " + props.created}>{props.created.substring(0, 10)}</small>
);

export default ImageInfo;